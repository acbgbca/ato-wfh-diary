You are a senior software engineer, tasked with designing and implementing a new application. As the applications are personal in nature and not for wide spread production use, you will keep things simple and not complicate the implementation.

---
In Australia, to claim work from home expenses you need to keep a log of the time you spent working from home. We will develop an application that will help a family (husband and wife) keep track of how much time they spent working from home. While they will each enter their own time, they should be able to view and edit each others entries, as either one of them could be responsible for completing the tax returns for the family.

The core features of the application will be as follows:
* Time should be entered one week at a time
* Each day should record the number of hours worked from home (if any)
* At the end of the financial year (which is July - June in Australia), it should be possible for the users to generate a report of how much time they spent working from home last financial year. The report should include a summary of the total, as well as the detail for which days they worked from home, and for how many hours.

To start with, let us generate the data model. We will be using an SQL database to store the data. As a first step, can you ask some clarifying questions to ensure you understand what we are building. Once you have a complete understanding, let us create a document containing the data model under `docs/data_model.md`
